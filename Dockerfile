ARG GO_VERSION="1.19.9"

FROM golang:${GO_VERSION}-alpine as deps
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM deps as apiBuilder
RUN GOOS=linux go build -o bin/api -ldflags "-s -w" ./cmd/api/main.go

FROM deps as dataloaderBuilder
RUN GOOS=linux go build -o bin/dataloader -ldflags "-s -w" ./cmd/dataloader/main.go

FROM alpine:3.18 as api
WORKDIR /root
EXPOSE 4000
COPY --from=apiBuilder /app/bin/api api
ENTRYPOINT ["/root/api"]

FROM alpine:3.18 as dataloader
WORKDIR /root
COPY --from=dataloaderBuilder /app/bin/dataloader dataloader
ENTRYPOINT ["/root/dataloader"]
