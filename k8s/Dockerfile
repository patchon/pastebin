FROM registry.access.redhat.com/ubi8/ubi-minimal:8.2

ENV yum_pkgs="iproute \
              git     \
              golang  \
              make    \
              python3 \
              sqlite"

ENV url_github="https://github.com/patchon/pastebin.git"
ENV url_pastebin="http://my-pastebin-url"
ENV url_replace="http://localhost:9999"
ENV pastebin_title="my-company-pastebin"

RUN microdnf install -y ${yum_pkgs}        &&\
    microdnf clean all                     &&\
    ln -s /usr/bin/python3 /usr/bin/python &&\
    pip-3 install pygments                 &&\
    git clone ${url_github}                &&\
    cd pastebin                            &&\
    make                                   &&\
    chmod -R g+wX /pastebin/               &&\
    sed -i "s|MyCompany|${pastebin_title}|"    config.json   &&\
    sed -i "s|${url_replace}|${url_pastebin}|" config.json   &&\
    sed -i "s|${url_replace}|${url_pastebin}|" assets/*.html &&\
    sed -i 's/localhost/0.0.0.0/' config.json

WORKDIR /pastebin

EXPOSE 9999/tcp
ENTRYPOINT ["./pastebin", "--debug"]
