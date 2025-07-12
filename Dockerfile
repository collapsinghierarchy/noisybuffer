# syntax=docker/dockerfile:1

############################
# 1. Build the static binary
############################
FROM golang:1.24.3-alpine AS build

WORKDIR /src
# copy only go.{mod,sum} first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build a fully-static Linux AMD64 binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /bin/noisybuffer ./cmd/noisybufferd

#############################################
# 2. Copy the binary into a tiny base image
#############################################
FROM gcr.io/distroless/static:nonroot

COPY --from=build /bin/noisybuffer /bin/noisybuffer
USER nonroot:nonroot        
EXPOSE 1234                 

ENTRYPOINT ["/bin/noisybuffer"]