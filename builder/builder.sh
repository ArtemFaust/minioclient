#!/bin/bash
if [[ $1 == ""|| $1 == "-h" ]];
then
  echo "./builder.sh build arm64|amd64"
  exit 0
fi

# Сборка пакета со статической линковкой
if [[ $1 == "build" ]];
then
  if [[ $2 == "" ]];
  then
    echo "platform ? arm64 or amd64 "
    exit 1
  else
    if [[ $2 == "amd64" ]];
    then
      docker build . --platform=linux/amd64 --tag minioclint_builder_amd64:latest
      docker run --platform linux/$2 --rm -ti  \
      -v ../src:/opt/src \
      -v ../dist:/opt/dist\
       minioclint_builder_amd64:latest bash ./entrypoint.sh
    elif [[ $2 == "arm64" ]];
    then
      docker build . --platform=linux/arm64 --tag minioclint_builder_arm64:latest
      docker run --platform linux/$2 --rm -ti \
      -v ../src:/opt/src \
      -v ../dist:/opt/dist\
      minioclint_builder_arm64:latest bash ./entrypoint.sh
    fi
  fi
fi