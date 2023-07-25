ARG GO_VERSION="1.19.9"

FROM golang:${GO_VERSION}-alpine as deps
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY api/ cmd/ common/ dataloader/ mongostore/ mq/ ./

FROM deps as apiBuilder
RUN GOOS=linux go build -o api -ldflags "-s -w" ./cmd/api/main.go

FROM deps as dataloaderBuilder
RUN GOOS=linux go build -o dataloader -ldflags "-s -w" ./cmd/dataloader/main.go

FROM alpine:3.18 as api
WORKDIR /root
EXPOSE 4000
COPY --from=apiBuilder /app/api api
ENTRYPOINT ["/root/api"]

FROM alpine:3.18 as dataloader
WORKDIR /root
COPY --from=dataloaderBuilder /app/dataloader dataloader
ENTRYPOINT ["/root/dataloader"]
