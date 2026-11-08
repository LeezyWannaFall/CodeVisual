FROM golang:1.26 AS go-build
WORKDIR /src
COPY main.go go.mod ./
RUN go build -o /out/server .

FROM python:3.12-slim AS runtime
WORKDIR /app
COPY worker/ /app/worker/
COPY --from=go-build /out/server /app/server
EXPOSE 8080
CMD ["/app/server"]
