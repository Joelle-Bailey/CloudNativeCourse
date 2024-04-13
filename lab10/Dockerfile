FROM golang:1.21-alpine AS build
WORKDIR /src/
COPY webserver.go /src/
RUN go mod init gitlab.com/Joelle-Bailey/cloudnativecourse
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /bin/webserver
FROM scratch
COPY --from=build /bin/webserver /bin/webserver
ENTRYPOINT ["/bin/webserver"]