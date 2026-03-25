FROM alpine:3.22

COPY helmdoc /usr/bin/helmdoc

WORKDIR /work

ENTRYPOINT ["/usr/bin/helmdoc"]
CMD ["version"]
