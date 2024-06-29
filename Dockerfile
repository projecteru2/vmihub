FROM golang:bookworm as gobuilder

WORKDIR /app
COPY . .

ENV GOPROXY https://goproxy.cn,direct
RUN sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list.d/debian.sources
RUN apt-get update
RUN apt-get install -y libcephfs-dev librbd-dev librados-dev build-essential
RUN make deps CN=1
RUN make build CN=1
RUN ./bin/vmihub --version

FROM debian:bookworm

RUN sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list.d/debian.sources && \
   apt-get update && \
   apt-get install -y libcephfs-dev librbd-dev librados-dev genisoimage qemu-utils
WORKDIR /app

COPY --from=gobuilder /app/bin/vmihub .

ENTRYPOINT [ "vmihub" ]
CMD ["--config", "config.toml", "server"]
