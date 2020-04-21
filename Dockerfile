FROM golang:1.14
WORKDIR /workspace/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o lamplighter *.go

FROM scratch
COPY --from=0 /workspace/lamplighter .
COPY --from=0 /workspace/secrets ./secrets
CMD ["./lamplighter"]
