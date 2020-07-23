#! /bin/bash

CSV='(.[0] | keys_unsorted) as $keys | ([$keys] + map([.[ $keys[] ]])) [] | @csv'

USER=''
PASSWD=''
INSTANCE="demo"
START=$(date -d "now - 30 days" +%Y-%m-%dT%H:%M:%S)
END=$(date +%Y-%m-%dT%H:%M:%S)
HOST="http://localhost:8090"
TYPE=""
DIRECTORY=$PWD
FORMAT=""

while getopts :f:d:u:p:s:e:i:t: OPT; do
  case $OPT in
    f)
    FORMAT=$OPTARG
    ;;
    d)
    DIRECTORY=$OPTARG
    # if [[ -f $DIRECTORY ]]; then
    #   echo "$DIRECTORY: file already exists and is regular file"
    #   exit 3
    # fi
    mkdir -p $DIRECTORY
    if [[ $? -ne 0 ]]; then
      echo "fail to create directory $DIRECTORY"
      exit 3
    fi
    ;;
    u)
    USER=$OPTARG
    ;;
    p)
    PASSWD=$OPTARG
    ;;
    s)
    START=$OPTARG
    ;;
    e)
    END=$OPTARG
    ;;
    i)
    INSTANCE=$OPTARG
    ;;
    t)
    TYPE=$OPTARG
    ;;
    *)
    echo "usage: $(basename $0) [-f format] [-d dir] [-s start] [-e end] [-i instance] [-t type] [-u user] [-p password] <url>"
    exit 1
    ;;
  esac
done

if [[ $(date -d "$START" +%s) -ge $(date -d "$END" +%s) ]]; then
  echo "invalid dates interval: $START - $END"
  exit 2
fi

retr_events() {
  HOST=$1

  DTSTART=$(date -d "$START" +%Y-%m-%dT%H:%M:%SZ)
  DTEND=$(date -d "$END" +%Y-%m-%dT%H:%M:%SZ)

  WHEN=$DTSTART
  while true; do
    mkdir -p "$DIRECTORY/$(date -d $WHEN +%Y)"
    FILE="$(date -d $WHEN +%j).csv"

    URL=$1'/api/archive/'"$INSTANCE"'/events/?start='"$WHEN"'&stop='"$(date -d "$WHEN + 1 day" +%Y-%m-%dT%H:%M:%SZ)"

    ACCEPT="application/json"
    if [[ $FORMAT == "csv" ]]; then
      ACCEPT="text/csv"
    fi
    curl -H "Accept: $ACCEPT" -u "$USER:$PASSWD" "$URL" 2> /dev/null > "$DIRECTORY/$(date -d $WHEN +%Y)/$FILE"

    WHEN=$(date -d "$WHEN + 1 days" +%Y-%m-%dT%H:%M:%SZ)
    if [[ $(date -d "$WHEN" +%s) -ge $(date -d "$END" +%s) ]]; then
      break;
    fi
  done
}

download_events() {
  HOST=$1

  DTSTART=$(date -d "$START" +%Y-%m-%dT%H:%M:%SZ)
  DTEND=$(date -d "$END" +%Y-%m-%dT%H:%M:%SZ)

  WHEN=$DTSTART
  while true; do
    mkdir -p "$DIRECTORY/$(date -d $WHEN +%Y)"
    FILE="$(date -d $WHEN +%j).csv"

    URL=$1'/api/archive/'"$INSTANCE"'/downloads/events/?start='"$WHEN"'&stop='"$(date -d "$WHEN + 1 day" +%Y-%m-%dT%H:%M:%SZ)"

    ACCEPT="application/json"
    if [[ $FORMAT == "csv" ]]; then
      ACCEPT="text/csv"
    fi
    curl -H "Accept: $ACCEPT" -u "$USER:$PASSWD" "$URL" 2> /dev/null > "$DIRECTORY/$(date -d $WHEN +%Y)/$FILE"

    WHEN=$(date -d "$WHEN + 1 days" +%Y-%m-%dT%H:%M:%SZ)
    if [[ $(date -d "$WHEN" +%s) -ge $(date -d "$END" +%s) ]]; then
      break;
    fi
  done
}

retr_cmd_history_logs() {
  FILTER='[.entry[] | {
    time: (.commandId.generationTime / 1000) | strftime("%Y-%m-%dT%H:%M:%SZ"),
    command: .commandId.commandName,
    code: .attr[]|select(.name=="CMDSRCCODE").value.stringValue,
    source: .attr[]|select(.name=="source").value.stringValue}]'
  HOST=$1

  DTSTART=$(date -d "$START" +%Y-%m-%dT%H:%M:%SZ)
  DTEND=$(date -d "$END" +%Y-%m-%dT%H:%M:%SZ)

  WHEN=$DTSTART
  while true; do
    mkdir -p "$DIRECTORY/$(date -d $WHEN +%Y)"
    FILE="$(date -d $WHEN +%j).csv"

    URL=$1'/api/archive/'"$INSTANCE"'/commands/?start='"$WHEN"'&stop='"$(date -d "$WHEN + 1 day" +%Y-%m-%dT%H:%M:%SZ)"

    if [[ $FORMAT == "csv" ]]; then
      curl -u "$USER:$PASSWD" "$URL" 2> /dev/null | jq -r "$FILTER" | jq -r "$CSV" > "$DIRECTORY/$(date -d $WHEN +%Y)/$FILE"
    else
      curl -u "$USER:$PASSWD" "$URL" 2> /dev/null > "$DIRECTORY/$(date -d $WHEN +%Y)/$FILE"
    fi

    WHEN=$(date -d "$WHEN + 1 days" +%Y-%m-%dT%H:%M:%SZ)
    if [[ $(date -d "$WHEN" +%s) -ge $(date -d "$END" +%s) ]]; then
      break;
    fi
  done
}

download_cmd_histpry_logs() {
  HOST=$1

  DTSTART=$(date -d "$START" +%Y-%m-%dT%H:%M:%SZ)
  DTEND=$(date -d "$END" +%Y-%m-%dT%H:%M:%SZ)

  WHEN=$DTSTART
  while true; do
    mkdir -p "$DIRECTORY/$(date -d $WHEN +%Y)"
    FILE="$(date -d $WHEN +%j).csv"

    URL=$1'/api/archive/'"$INSTANCE"'/commands/?start='"$WHEN"'&stop='"$(date -d "$WHEN + 1 day" +%Y-%m-%dT%H:%M:%SZ)"
    curl -u "$USER:$PASSWD" "$URL" 2> /dev/null > "$DIRECTORY/$(date -d $WHEN +%Y)/$FILE"

    WHEN=$(date -d "$WHEN + 1 days" +%Y-%m-%dT%H:%M:%SZ)
    if [[ $(date -d "$WHEN" +%s) -ge $(date -d "$END" +%s) ]]; then
      break;
    fi
  done
}

shift $(($OPTIND - 1))
if [[ -n $1 ]]; then
  HOST=$1
fi
case "$TYPE" in
  "cmdhist")
  retr_cmd_history_logs $HOST
  ;;
  "rawcmdhist")
  download_cmd_history_logs $HOST
  ;;
  "events")
  retr_events $HOST
  ;;
  "rawevents")
  retr_events $HOST
  ;;
  *)
  echo "unsupported request type: $TYPE"
  exit 2
  ;;
esac
