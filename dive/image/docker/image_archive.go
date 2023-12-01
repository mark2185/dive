package docker

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/wagoodman/dive/dive/filetree"
	"github.com/wagoodman/dive/dive/image"
)

func hasSuffix(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.HasSuffix(haystack, n) {
			return true
		}
	}
	return false
}

type ImageArchive struct {
	manifest manifest
	config   config
	layerMap map[string]*filetree.FileTree
}

func NewImageArchive(tarFile io.ReadCloser) (*ImageArchive, error) {
	img := &ImageArchive{
		layerMap: map[string]*filetree.FileTree{},
	}

	// store discovered json files in a map so we can read the image in one pass
	jsonFiles := make(map[string][]byte)

	tarReader := tar.NewReader(tarFile)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// name of file entry
		name := header.Name

		// some layer tars can be relative layer symlinks to other layer tars
		// TODO: what happens with other types?
		// TODO: log when a different type appears
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeReg {
			if strings.HasSuffix(name, ".json") || strings.HasPrefix(name, "sha256:") {
				// metadata files
				fileBuffer, err := io.ReadAll(tarReader)
				if err != nil {
					return img, err
				}
				jsonFiles[name] = fileBuffer
			} else if hasSuffix(name, ".tar", ".tar.gz", ".tgz") {
				// actual tar with files
				layerReader := tar.NewReader(tarReader)
				if hasSuffix(name, ".tar.gz", ".tgz") {
					// have to decompress
					gz, err := gzip.NewReader(tarReader)
					if err != nil {
						return img, err
					}
					layerReader = tar.NewReader(gz)
				}
				tree, err := processTarLayer(name, layerReader)
				if err != nil {
					return img, err
				}

				// add the layer to the image
				img.layerMap[tree.Name] = tree
			}
			// if it's neither, then it's probably a VERSION file or something else
			// TODO: investigate the OCI standard to see which other files could it be
		}
	}

	manifestContent, exists := jsonFiles["manifest.json"]
	if !exists {
		return img, fmt.Errorf("could not find image manifest")
	}

	img.manifest = newManifest(manifestContent)

	configContent, exists := jsonFiles[img.manifest.ConfigPath]
	if !exists {
		return img, fmt.Errorf("could not find image config")
	}

	img.config = newConfig(configContent)

	return img, nil
}

func processTarLayer(name string, reader *tar.Reader) (*filetree.FileTree, error) {
	tree := filetree.NewFileTree()
	tree.Name = name

	fileInfos, err := getFileList(reader)
	if err != nil {
		return nil, err
	}

	for _, element := range fileInfos {
		tree.FileSize += uint64(element.Size)

		_, _, err := tree.AddPath(element.Path, element)
		if err != nil {
			return nil, err
		}
	}

	// TODO: spin off in a separate thread
	tree.Root.CalculateSize()
	return tree, nil
}

// takes a layer as .tar and returns all files
func getFileList(tarReader *tar.Reader) ([]filetree.FileInfo, error) {
	var files []filetree.FileInfo

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// always ensure relative path notations are not parsed as part of the filename
		name := path.Clean(header.Name)
		if name == "." {
			continue
		}

		// TODO: why are these unexpected?
		switch header.Typeflag {
		case tar.TypeXGlobalHeader:
			return nil, fmt.Errorf("unexpected tar file: (XGlobalHeader): type=%v name=%s", header.Typeflag, name)
		case tar.TypeXHeader:
			return nil, fmt.Errorf("unexpected tar file (XHeader): type=%v name=%s", header.Typeflag, name)
		default:
			files = append(files, filetree.NewFileInfoFromTarHeader(tarReader, header, name))
		}
	}
	return files, nil
}

func (img *ImageArchive) ToImage() (*image.Image, error) {
	trees := []*filetree.FileTree{}

	// build the content tree
	for _, treeName := range img.manifest.LayerTarPaths {
		tr, exists := img.layerMap[treeName]
		if !exists {
			return nil, fmt.Errorf("could not find '%s' in parsed layers", treeName)
		}
		trees = append(trees, tr)
	}

	// build the layers array
	layers := []*image.Layer{}

	// note that the engineResolver config stores images in reverse chronological order, so iterate backwards through layers
	// as you iterate chronologically through history (ignoring history items that have no layer contents)
	// Note: history is not required metadata in a docker image!
	histIdx := 0
	for idx, tree := range trees {
		// ignore empty layers, we are only observing layers with content
		historyObj := historyEntry{
			CreatedBy: "(missing)",
		}
		for nextHistIdx := histIdx; nextHistIdx < len(img.config.History); nextHistIdx++ {
			if !img.config.History[nextHistIdx].EmptyLayer {
				histIdx = nextHistIdx
				break
			}
		}
		if histIdx < len(img.config.History) && !img.config.History[histIdx].EmptyLayer {
			historyObj = img.config.History[histIdx]
			histIdx++
		}

		historyObj.Size = tree.FileSize

		dockerLayer := layer{
			history: historyObj,
			index:   idx,
			tree:    tree,
		}
		layers = append(layers, dockerLayer.ToLayer())
	}

	return &image.Image{
		Trees:  trees,
		Layers: layers,
	}, nil
}
