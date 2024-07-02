FROM golang:bookworm as gobuilder

WORKDIR /app
COPY . .

ENV GOPROXY https://goproxy.cn,direct
RUN apt update -y \
    apt install -y libcephfs-dev librbd-dev librados-dev build-essential
RUN make deps CN=1
RUN make build CN=1
RUN ./bin/vmihub --version

FROM debian:bookworm
RUN apt update -y && \
   apt install -y libcephfs-dev librbd-dev librados-dev genisoimage qemu-utils
WORKDIR /app

COPY --from=gobuilder /app/bin/vmihub .

ENTRYPOINT [ "vmihub" ]
CMD ["--config", "config.toml", "server"]
