FROM golang:1.16
WORKDIR /workspace/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o lamplighter -v ./cmd/lamplighter

FROM scratch
WORKDIR /root/
COPY --from=0 /workspace/lamplighter /root/lamplighter
COPY --from=0 /workspace/secrets /root/secrets
COPY --from=0 /usr/local/go/lib/time/zoneinfo.zip /zoneinfo.zip

ARG TZ=EST5EDT
ENV TZ=${TZ}
ENV ZONEINFO=/zoneinfo.zip

CMD ["./lamplighter"]
