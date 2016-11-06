FROM iron/go:dev
 
RUN mkdir -p /go/src/github.com/Masterminds
ENV GOPATH=/go
RUN cd /go/src/github.com/Masterminds && git clone https://github.com/Masterminds/glide.git && cd glide && go build
RUN cp /go/src/github.com/Masterminds/glide/glide /bin

ENTRYPOINT ["glide"]