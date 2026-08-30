FROM scratch
ARG TARGETPLATFORM
ENTRYPOINT ["/usr/bin/afon"]
COPY ${TARGETPLATFORM}/afon /usr/bin/
