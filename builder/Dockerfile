FROM golang:latest
COPY entrypoint.sh /opt/entrypoint.sh
COPY go.mod /opt/go.mod
COPY go.sum /opt/go.sum
RUN chmod +x /opt/entrypoint.sh
WORKDIR /opt
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
CMD ["entrypoint.sh"]