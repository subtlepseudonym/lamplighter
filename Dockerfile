FROM golang:1.19 as build
WORKDIR /workspace/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o lamplighter -v ./cmd/lamplighter

FROM scratch
COPY --from=build /workspace/lamplighter /lamplighter
COPY --from=build /usr/local/go/lib/time/zoneinfo.zip /zoneinfo.zip
COPY --from=tarampampam/curl:7.78.0 /bin/curl /curl

ARG TZ=EST5EDT
ENV TZ=${TZ}
ENV ZONEINFO=/zoneinfo.zip

CMD ["/lamplighter"]
