FROM golang:1.27.0 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY cmd ./cmd
COPY configs ./configs
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/server ./server
COPY --from=builder /src/configs/configs.webserver.yml ./configs/configs.webserver.yml

EXPOSE 8080

ENTRYPOINT ["./server", "start"]
