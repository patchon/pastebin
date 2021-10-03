#!/bin/bash

# Wrapper that enables us to configure the pastebin from envs on the fly.

config="/pastebin/config.json"
assets="/pastebin/assets/*.html"
replace_prt="9999"
replace_url="http://localhost:9999"
replace_str="MyCompany"
default_port="8443"
PB_PORT=${PB_PORT:=default}

if [[ -z "${PB_NAME}" ]];then
  echo "[ ERR ] : ENV PB_NAME is not set"
  exit 1
fi

if [[ -z "${PB_URL}" ]];then
  echo "[ ERR ] : ENV PB_URL is not set"
  exit 1
fi

if [[ "${PB_PORT}" == "default" ]];then
  PB_PORT=${default_port}
  echo "[ NOTICE ] : ENV PB_PORT is not set, using default port ${default_port}"
fi

echo "[ INFO ] : PB_NAME is set to ${PB_NAME}"
echo "[ INFO ] : PB_URL is set to ${PB_URL}"
echo "[ INFO ] : PB_PORT is set to ${PB_PORT}"

# Replace config
sed -i -e "s|${replace_str}|${PB_NAME}| " \
       -e "s|${replace_url}|${PB_URL}|  " \
       -e "s|${replace_prt}|${PB_PORT}| " \
       -e 's/localhost/0.0.0.0/' "${config}"

# Replace hardcoded url in assetes, pew pew
sed -i -e "s|${replace_url}|${PB_URL}|" ${assets}

cd /pastebin
./pastebin --debug
