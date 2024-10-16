#!/usr/bin/zsh

cd $DEVEX_PATH
last_updated_at=$(git log -1 --format=%cd --date=unix)
git pull

for file in $DEVEX_PATH/migrations/*.sh; do
  filename=$(basename "$file")
  migrate_at="${filename%.sh}"

  if [ $migrate_at -gt $last_updated_at ]; then
    echo "Running migration for $migrate_at"
    source $file
  fi
done

cd -
gum spin --spinner globe --title "Update has completed!" -- sleep 3
clear
source $DEVEX_PATH/bin/devex.sh
