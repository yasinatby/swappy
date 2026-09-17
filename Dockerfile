FROM golang:1.27-alpine AS build

WORKDIR /src

COPY backend/go.mod ./
RUN go mod download

COPY backend/ ./

RUN CGO_ENABLED=0 go build -o /out/swappy-backend ./cmd/swappy-backend

FROM alpine:latest

WORKDIR /app

COPY --from=build /out/swappy-backend ./swappy-backend

ENV SWAPPY_HOST=0.0.0.0
ENV SWAPPY_PORT=8080

EXPOSE 8080

CMD ["./swappy-backend"]