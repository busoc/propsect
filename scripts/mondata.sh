#! /bin/bash


diskUsage() {
  dtstart=$START
  dtend=$END
  dat=$1
  base=$2

  size=0
  until [[ $dtstart -ge $dtend ]]; do
    path="$BASE/$base/RealTime/$(/bin/date -d @$dtstart +%Y/%j)"

    dtstart=$(/bin/date -d @$dtstart +%Y-%m-%d)
    dtstart=$(/bin/date -d "$dtstart + 1 days" +%s)
    if [[ ! -d $path ]]; then
      continue
    fi
    z=$(/usr/bin/du -b -s $path | cut -f 1)
    size=$(($size+$z))
  done

  if [[ $size -gt 0 ]]; then
    /bin/echo $PAYLOAD $MODE $dat $size #$(numfmt --to=si $size)
  fi
}

MODE=Ops
PAYLOAD=""
START=$(/bin/date -d "yesterday" +%s)
END=$(/bin/date +%s)

while getopts m:p:s:e: OPT; do
  case $OPT in
    m)
      MODE=$OPTARG
      ;;
    p)
      PAYLOAD=$OPTARG
      ;;
    s)
      START=$(/bin/date -d $OPTARG +%s)
      ;;
    e)
      END=$(/bin/date -d $OPTARG +%s)
      ;;
    *)
      exit 1
      ;;
  esac
done
shift $(($OPTIND - 1))

BASE="$1/$PAYLOAD/Archive/$MODE"
if [[ ! -d $BASE ]]; then
  /bin/echo "$BASE: not a directory" 1>&2
  exit 2
fi

export BASE
export START
export END
export MODE
export PAYLOAD
export -f diskUsage

/bin/echo "#$(/bin/date +%Y/%j)"
/usr/bin/parallel diskUsage {1} {2} ::: TM PP HRDL :::+ "PathTM/PTH" "ProcessedData/PDH" "HRDL/2" 2> /dev/null
