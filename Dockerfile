FROM alpine:3.18

# ARG DOCKER_CLI_VERSION=${DOCKER_CLI_VERSION}
# RUN wget -O- https://download.docker.com/linux/static/stable/$(uname -m)/docker-${DOCKER_CLI_VERSION}.tgz | \
    # tar -xzf - docker/docker --strip-component=1 && \
    # mv docker /usr/local/bin

COPY dive /usr/local/bin/

RUN echo "contents" >> /usr/local/bin/file
RUN echo "update" >> /usr/local/bin/file
RUN rm /usr/local/bin/file

RUN ARG=1
RUN ARG=2
RUN ARG=3
RUN ARG=4
RUN ARG=5
RUN ARG=6
RUN ARG=7
RUN ARG=8
RUN ARG=9
RUN ARG=10
RUN ARG=11
RUN ARG=12
RUN ARG=13
RUN ARG=14
RUN ARG=15
RUN ARG=16
RUN ARG=17
RUN ARG=18
RUN ARG=19
RUN ARG=20
RUN ARG=21
RUN ARG=22
RUN ARG=23
RUN ARG=24
RUN ARG=25
RUN ARG=26
RUN ARG=27
RUN ARG=9
RUN ARG=10
RUN ARG=11
RUN ARG=12
RUN ARG=13
RUN ARG=14
RUN ARG=15
RUN ARG=16
RUN ARG=17
RUN ARG=18
RUN ARG=19
RUN ARG=20
RUN ARG=21
RUN ARG=22
RUN ARG=23
RUN ARG=24
RUN ARG=25
RUN ARG=26

ENTRYPOINT ["/usr/local/bin/dive"]
