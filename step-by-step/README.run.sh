#!/bin/bash

while IFS='' read -r line || [[ -n "$line" ]]; do
  if [[ "$line" =~ '##' ]]
  then
    cmd="$(echo $line | sed 's/^##[[:space:]]*//')"
    echo "[$cmd] Run..."
    xterm -T "$cmd" -e go run "$cmd" &
    while :
    do
      started="$(netstat -lnt | grep 8080 | wc -l)"
      if [[ "$started" = '0' ]]
      then
        sleep 1
      else
        break
      fi
    done
    echo "[$cmd] Call..."
    while IFS='' read -r line || [[ -n "$line" ]]; do
      if [[ "$line" = '```' ]]
      then
        break
      fi
    done
    while IFS='' read -r line || [[ -n "$line" ]]; do
      if [[ "$line" = '```' ]]
      then
        break
      fi
      line="$(echo "$line" | sed 's-#.*--')"
      echo ">> $line"
      curl -XPOST http://localhost:8080/gql -H 'Content-Type: application/graphql' -d "$line"
      echo
    done
    echo "[$cmd] Kill..."
    kill $(jobs -p)
    echo "[$cmd] Wait..."
    wait
  fi
done < README.md
