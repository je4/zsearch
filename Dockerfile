FROM alpine:3

WORKDIR /app
ADD zsearch /app
RUN apk add --no-cache openssl
RUN openssl genrsa -traditional -out private-key.pem
RUN mkdir /app/cache
RUN mkdir /opt/zsearch
ADD search.toml /opt/zsearch/search.toml
ENTRYPOINT ["./zsearch", "-cfg", "/opt/zsearch/search.toml"]
EXPOSE 443
