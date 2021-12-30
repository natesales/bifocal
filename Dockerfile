FROM alpine
COPY bifocal /usr/bin/bifocal
ENTRYPOINT ["/usr/bin/bifocal"]