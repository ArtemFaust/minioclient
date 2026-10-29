#!/bin/bash
echo -e "\033[33m==============Select binary==============\033[0m"
if [[ $(arch) == "x86_64" ]];
then
    EXEC="$(pwd)/linux/amd64/minioclient"
    echo $EXEC
elif [[ $(arch) == "aarch64" ]];
then
    EXEC="$(pwd)/linux/aarch64/minioclient"
    echo $EXEC
fi
echo -e "\033[33m==============End select==============\033[0m"

if [[ ! -x "$EXEC" ]]; then
    echo -e "\033[31;1m--- Error: binary not found $EXEC ---\033[0m"
    exit 1
fi

# Run master service
function runmaster {
    if [[ $DEBUG == "true" ]];
    then
        $EXEC -serve -master -d -masterport $MASTERPORT
        exit $?
    fi
    if [[ $DEBUG == "false" ]];
    then
        $EXEC -serve -master -masterport $MASTERPORT
        exit $?
    fi
}

# Run slave service
function runslave {
    if [[ $DEBUG == "true" ]];
    then
        $EXEC -serve -slave -masterendpoint $MASTERENDPOINT -d
        exit $?
    fi
    if [[ $DEBUG == "false" ]];
    then
        $EXEC -serve -slave -masterendpoint $MASTERENDPOINT
        exit $?
    fi
}

if [[ $MASTER == "true" ]];
then
    runmaster
fi

if [[ $SLAVE == "true" ]];
then
    runslave
fi