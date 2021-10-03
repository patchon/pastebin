FROM registry.redhat.io/ubi8/ubi-minimal:8.3

ENV yum_pkgs="iproute \
              git     \
              golang  \
              make    \
              python3 \
              sqlite  \
              vim"

ENV url_pastebin="https://github.com/patchon/pastebin.git"

COPY contrib/entrypoint.sh /
RUN chmod +x /entrypoint.sh                &&\
    microdnf install -y ${yum_pkgs}        &&\
    microdnf clean all                     &&\
    ln -s /usr/bin/python3 /usr/bin/python &&\
    pip-3 install pygments                 &&\
    echo "Cloning"                         &&\
    git clone ${url_pastebin}              &&\
    cd pastebin                            &&\
    make                                   &&\
    chmod -R g+w /pastebin/

EXPOSE 8443/tcp
ENTRYPOINT ["/entrypoint.sh"]
