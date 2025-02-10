FROM golang:1.23-alpine AS builder

WORKDIR /workspace
RUN apk update && apk add --no-cache make git bash
COPY ../go.mod ../go.sum ./
RUN go env -w GOPROXY="https://goproxy.io,direct"
RUN go mod download -x
COPY .. .
RUN make opea-downloader



FROM python:3.12.9-alpine3.21

LABEL authors="airren"

# Install huggingface-cli
RUN pip install -U "huggingface_hub[cli]"
RUN huggingface-cli --help
COPY --from=builder /workspace/bin/opea-downloader /usr/bin/

ENTRYPOINT ["apk", "update"]