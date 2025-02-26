# Copyright 2024 Redpanda Data, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.24 AS build

ENV CGO_ENABLED=0
ENV GOOS=linux
RUN #useradd -u 10001 proxy

WORKDIR /go/src/
# Update dependencies: On unchanged dependencies, cached layer will be reused
COPY go.* /go/src/
RUN go mod download

# Build
COPY . /go/src/
# Tag timetzdata required for busybox base image:
# https://github.com/benthosdev/benthos/issues/897

RUN go build .

# Pack
FROM busybox AS package

WORKDIR /

#COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
#COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /go/src/sr-fix-proxy /
#COPY ./config/docker.yaml /connect.yaml

#USER proxy

EXPOSE 8081

ENTRYPOINT ["/sr-fix-proxy"]

CMD ["-config", "/config.yaml"]
