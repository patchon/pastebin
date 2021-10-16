FROM registry.access.redhat.com/ubi8/ubi-minimal:8.4

ENV yum_pkgs="git     \
              golang  \
              make    \
              python3 \
              sqlite  \
              "

ENV url_pastebin="https://github.com/patchon/pastebin.git"

RUN microdnf install -y ${yum_pkgs}        &&\
    microdnf clean all                     &&\
    ln -s /usr/bin/python3 /usr/bin/python &&\
    pip-3 install pygments                 &&\
    git clone ${url_pastebin}              &&\
    cd pastebin                            &&\
    make                                   &&\
    chmod -R g+w /pastebin/

EXPOSE 8443/tcp
WORKDIR /pastebin
ENTRYPOINT ["./pastebin", "--debug"]
