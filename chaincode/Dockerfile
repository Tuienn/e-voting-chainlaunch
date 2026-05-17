FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/chaincode .

FROM alpine:3.22

RUN addgroup -S chaincode && adduser -S chaincode -G chaincode

WORKDIR /app

COPY --from=builder /out/chaincode /app/chaincode

ENV CHAINCODE_SERVER_ADDRESS=0.0.0.0:9999

EXPOSE 9999

USER chaincode

ENTRYPOINT ["/app/chaincode"]
