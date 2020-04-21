FROM golang:1.14

WORKDIR /workspace
COPY . .
RUN go mod download
RUN GOOS=linux go build -a -o lamplighter *.go

FROM scratch
COPY --from=0 /workspace/lamplighter .
COPY --from=0 /workspace/home.loc .
COPY --from=0 /workspace/lifx.token .
CMD ["/lamplighter"]
