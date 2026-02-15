#!/bin/sh
set -e

# Copy default config files if not exist
if [ ! -f /app/conf/config-base.json ]; then
    cp /app/conf-default/config-base.json /app/conf/
fi

if [ ! -f /app/conf/seelog.xml ]; then
    cp /app/conf-default/seelog.xml /app/conf/
fi

# Copy default database if not exists
if [ ! -f /app/db/database-base.db ]; then
    cp /app/db-default/database-base.db /app/db/
fi

exec ./smartping
