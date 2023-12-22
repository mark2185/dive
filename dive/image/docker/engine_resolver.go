package docker

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	cliconfig "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/connhelper"
	ddocker "github.com/docker/cli/cli/context/docker"
	ctxstore "github.com/docker/cli/cli/context/store"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"

	"github.com/wagoodman/dive/dive/image"
)

type engineResolver struct{}

func NewResolverFromEngine() *engineResolver {
	return &engineResolver{}
}

func (r *engineResolver) Fetch(id string) (*image.Image, error) {
	reader, err := r.fetchArchive(id)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	img, err := NewImageArchive(reader)
	if err != nil {
		return nil, err
	}
	return img.ToImage()
}

func (r *engineResolver) Build(args []string) (*image.Image, error) {
	id, err := buildImageFromCli(args)
	if err != nil {
		return nil, err
	}
	return r.Fetch(id)
}

// determineDockerHost tries to determine the docker host that we should connect to
// in the following order of decreasing precedence:
//   - value of the "DOCKER_HOST" environment variable
//   - host retrieved from the current context (specified via DOCKER_CONTEXT)
//   - "default docker host" for the host operating system, otherwise
func determineDockerHost() (string, error) {
	// if the docker host is explicitly set via the "DOCKER_HOST" environment variable,
	// then it's a no-brainer :shrug:
	if host := os.Getenv("DOCKER_HOST"); len(host) > 0 {
		return host, nil
	}

	currentContext := os.Getenv("DOCKER_CONTEXT")
	if len(currentContext) == 0 {
		dockerConfigDir := cliconfig.Dir()
		if _, err := os.Stat(dockerConfigDir); err != nil {
			return "", err
		}
		cf, err := cliconfig.Load(dockerConfigDir)
		if err != nil {
			return "", err
		}
		currentContext = cf.CurrentContext
	}
	if len(currentContext) == 0 {
		// if a docker context is neither specified via the "DOCKER_CONTEXT" environment variable nor via the
		// $HOME/.docker/config file, then we fall back to connecting to the "default docker host" meant for
		// the host operating system
		if runtime.GOOS == "windows" {
			return "npipe:////./pipe/docker_engine", nil
		} else {
			return "unix:///var/run/docker.sock", nil
		}
	}

	storeConfig := ctxstore.NewConfig(
		func() interface{} { return &ddocker.EndpointMeta{} },
		ctxstore.EndpointTypeGetter(ddocker.DockerEndpoint, func() interface{} { return &ddocker.EndpointMeta{} }),
	)

	st := ctxstore.New(cliconfig.ContextStoreDir(), storeConfig)
	md, err := st.GetMetadata(currentContext)
	if err != nil {
		return "", err
	}
	dockerEP, ok := md.Endpoints[ddocker.DockerEndpoint]
	if !ok {
		return "", err
	}
	dockerEPMeta, ok := dockerEP.(ddocker.EndpointMeta)
	if !ok {
		return "", fmt.Errorf("expected docker.EndpointMeta, got %T", dockerEP)
	}

	if dockerEPMeta.Host != "" {
		return dockerEPMeta.Host, nil
	}

	// we might end up here if the context was created with the `host` set to an empty value (i.e. '')
	// for example:
	// ```sh
	// docker context create foo --docker "host="
	// ```
	// in such a scenario, we mimic the `docker` cli and try to connect to the "default docker host"
	if runtime.GOOS == "windows" {
		return "npipe:////./pipe/docker_engine", nil
	} else {
		return "unix:///var/run/docker.sock", nil
	}
}

func (r *engineResolver) fetchArchive(id string) (io.ReadCloser, error) {
	var err error
	var dockerClient *client.Client

	// pull the engineResolver if it does not exist
	ctx := context.Background()

	host := os.Getenv("DOCKER_HOST")
	var clientOpts []client.Opt

	switch strings.Split(host, ":")[0] {
	case "ssh":
		helper, err := connhelper.GetConnectionHelper(host)
		if err != nil {
			fmt.Println("docker host", err)
		}
		clientOpts = append(clientOpts, func(c *client.Client) error {
			httpClient := &http.Client{
				Transport: &http.Transport{
					DialContext: helper.Dialer,
				},
			}
			return client.WithHTTPClient(httpClient)(c)
		})
		clientOpts = append(clientOpts, client.WithHost(helper.Host))
		clientOpts = append(clientOpts, client.WithDialContext(helper.Dialer))

	default:
		dockerHost, err := determineDockerHost()
		if err != nil {
			fmt.Printf("Could not determine host %v", err)
		}

		if os.Getenv("DOCKER_TLS_VERIFY") != "" && os.Getenv("DOCKER_CERT_PATH") == "" {
			os.Setenv("DOCKER_CERT_PATH", "~/.docker")
		}

		clientOpts = append(clientOpts, client.FromEnv)
		clientOpts = append(clientOpts, client.WithHost(dockerHost))
	}

	clientOpts = append(clientOpts, client.WithAPIVersionNegotiation())

	dockerClient, err = client.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, err
	}
	_, _, err = dockerClient.ImageInspectWithRaw(ctx, id)
	if err != nil {
		// don't use the API, the CLI has more informative output
		fmt.Println("Handler not available locally. Trying to pull '" + id + "'...")
		err = runDockerCmd("pull", id)
		if err != nil {
			return nil, err
		}
	}

	readCloser, err := dockerClient.ImageSave(ctx, []string{id})
	if err != nil {
		return nil, err
	}

	return readCloser, nil
}
