FROM alpine:latest

COPY ./quartermaster /

EXPOSE 9090
CMD ["./quartermaster"]
