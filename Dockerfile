FROM golang:1.14
WORKDIR /workspace/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o lamplighter *.go

FROM scratch
WORKDIR /root/
COPY --from=0 /workspace/lamplighter /root/lamplighter
COPY --from=0 /workspace/secrets /root/secrets
COPY --from=0 /usr/local/go/lib/time/zoneinfo.zip /zoneinfo.zip

ARG TZ=EST5EDT
ENV TZ=${TZ}
ENV ZONEINFO=/zoneinfo.zip

CMD ["./lamplighter"]
