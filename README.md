# ticker

## development

```
go run main.go -config config.yml.dist
```

## configuration

```
# listen binds ticker to specific address and port
listen: "localhost:8080"
# log_level sets log level for logrus
log_level: "error"
# initiator is the email for the first admin user (see password in logs)
initiator: "admin@systemli.org"
# database is the path to the bolt file
database: "ticker.db"
# secret used for JSON Web Tokens
secret: "slorp-panfil-becall-dorp-hashab-incus-biter-lyra-pelage-sarraf-drunk"
```

## testing

```
go test ./... -cover
```
