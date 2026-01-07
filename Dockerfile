# HINT: prepare a docker multi platform build
#docker buildx create --use --platform=linux/arm64,linux/amd64 --name multi-platform-builder
#docker buildx inspect --bootstrap
#docker buildx build --platform=linux/arm64,linux/amd64 --push -t ghcr.io/danpaul81/px-deploy:dev .
FROM --platform=$BUILDPLATFORM golang:1.25-alpine3.22 AS build
RUN mkdir -p /linux/amd64
RUN mkdir -p /linux/arm64
RUN wget -P / https://releases.hashicorp.com/terraform/1.9.8/terraform_1.9.8_linux_amd64.zip
RUN wget -P / https://releases.hashicorp.com/terraform/1.9.8/terraform_1.9.8_linux_arm64.zip
RUN unzip /terraform_1.9.8_linux_amd64.zip -d /linux/amd64
RUN unzip /terraform_1.9.8_linux_arm64.zip -d /linux/arm64
RUN wget -P / https://github.com/vmware/govmomi/releases/download/v0.52.0/govc_Linux_x86_64.tar.gz
RUN wget -P / https://github.com/vmware/govmomi/releases/download/v0.52.0/govc_Linux_arm64.tar.gz
RUN tar -xzf /govc_Linux_x86_64.tar.gz -C /linux/amd64
RUN tar -xzf /govc_Linux_arm64.tar.gz -C /linux/arm64
RUN mkdir -p /root/go/src/px-deploy
COPY go.mod go.sum *.go /root/go/src/px-deploy/
ARG TARGETOS TARGETARCH TARGETPLATFORM
RUN cd /root/go/src/px-deploy; GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /$TARGETPLATFORM/px-deploy
COPY terraform /px-deploy/terraform

FROM --platform=$TARGETPLATFORM alpine:3.22
RUN apk add --no-cache openssh-client-default bash rsync
RUN echo ServerAliveInterval 300 >/etc/ssh/ssh_config
RUN echo ServerAliveCountMax 2 >>/etc/ssh/ssh_config
RUN echo TCPKeepAlive yes >>/etc/ssh/ssh_config
ARG TARGETPLATFORM
COPY --from=build /$TARGETPLATFORM/terraform /usr/bin/terraform
COPY --from=build /$TARGETPLATFORM/govc /usr/bin/govc
COPY --from=build /$TARGETPLATFORM/px-deploy /root/go/bin/px-deploy
COPY assets /px-deploy/assets
COPY scripts /px-deploy/scripts
COPY templates /px-deploy/templates
COPY infra /px-deploy/infra
COPY defaults.yml /px-deploy/versions.yml
COPY VERSION /
COPY --from=build /px-deploy/terraform/aws /px-deploy/terraform/aws 
COPY --from=build /px-deploy/terraform/azure /px-deploy/terraform/azure
COPY --from=build /px-deploy/terraform/gcp /px-deploy/terraform/gcp
COPY --from=build /px-deploy/terraform/vsphere /px-deploy/terraform/vsphere
RUN terraform -chdir=/px-deploy/terraform/aws/ init
RUN terraform -chdir=/px-deploy/terraform/azure/ init
RUN terraform -chdir=/px-deploy/terraform/gcp/ init
RUN terraform -chdir=/px-deploy/terraform/vsphere/ init
 
