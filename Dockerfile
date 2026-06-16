#########################################################################################################
# Backend Build (Go)
#########################################################################################################
FROM golang:1.21 as backend-build

WORKDIR /go/src/github.com/fastenhealth/fasten-onprem
COPY . .

RUN --mount=type=cache,target=/tmp/lock,sharing=locked \
    go mod vendor \
    && go install github.com/golang/mock/mockgen@v1.6.0 \
    && go generate ./... \
    && go build -ldflags "-extldflags=-static" -tags "static" -o /go/bin/fasten ./backend/cmd/fasten/

# create folder structure
RUN mkdir -p /opt/fasten/db \
  && mkdir -p /opt/fasten/web \
  && mkdir -p /opt/fasten/config

#########################################################################################################
# Distribution Build (Distroless)
#########################################################################################################
FROM gcr.io/distroless/static-debian11

EXPOSE 8080
WORKDIR /opt/fasten/
COPY --from=backend-build  /opt/fasten/ /opt/fasten/
COPY --from=backend-build /go/bin/fasten /opt/fasten/fasten
COPY LICENSE.md /opt/fasten/LICENSE.md
COPY config.yaml /opt/fasten/config/config.yaml

CMD ["/opt/fasten/fasten", "start", "--config", "/opt/fasten/config/config.yaml"]
