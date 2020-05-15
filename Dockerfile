# ----------------------------------------------------------------------------------------
# Image: Builder
# ----------------------------------------------------------------------------------------
FROM golang:1.14-alpine as builder

# setup the environment
ENV TZ=Europe/Berlin

# install dependencies
RUN apk --update --no-cache add git gcc musl-dev tzdata
WORKDIR /work
ADD ./ ./

# build the go binary
RUN go build -ldflags \
        '-X "main.BuildTime='$(date -Iminutes)'" \
         -X "main.GitCommit='$(git rev-parse --short HEAD)'" \
         -X "main.GitBranch='$(git rev-parse --abbrev-ref HEAD)'" \
         -s -w' \
         -v -o /tmp/kallax .

RUN chown nobody:nobody /tmp/kallax && \
    chmod +x /tmp/kallax

# ----------------------------------------------------------------------------------------
# Image: Deployment
# ----------------------------------------------------------------------------------------
FROM alpine:latest
MAINTAINER Maximilian Pachl <m@ximilian.info>

# add relevant files to container
COPY --from=builder /tmp/kallax /usr/sbin/kallax

EXPOSE 5353/udp
EXPOSE 9800
CMD /usr/sbin/kallax
