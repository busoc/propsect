#! /bin/bash

function splitTM() {
  which tmcat &> /dev/null
  if [ $? -ne 0 ]; then
    echo "tmcat is not installed"
    exit 432
  fi

  workdir=$(realpath $2)
  datadir=$(realpath $1)
  shift 2

  if [ ${#} -eq 0 ]; then
    return 0
  fi

  parallel --will-cite --jobs 4 tmcat take -p {1} -d 24h $workdir/%P/%Y/%0J.dat $datadir ::: $@
  if [ $? -ne 0 ]; then
    return 1
  fi
}

function splitPP() {
  echo "segregate PP: not yet implemented"
  exit 255
}

function splitVMU() {
  echo "segregate HR: not yet implemented"
  exit 255
}

which parallel > /dev/null
if [ $? -ne 0 ]; then
  echo "parallel is not installed"
  exit 432
fi

JOBS=4
TYPE=TM
# DATADIR: archive where data will be read (hrdp archive)
# WORKDIR: archive where data will be written (busoc archive)
DATADIR=$TMPDIR
WORKDIR=$TMPDIR

while getopts :t:d:w: OPT; do
  case $OPT in
    j)
      if [ -n $OPTARG ] && [ $OPTARG -eq $OPTARG ] && [ $OPTARG -ne 0 ]; then
        JOBS=$OPTARG
      fi
      ;;
    t)
      TYPE=$OPTARG
      ;;
    w)
      WORKDIR=$OPTARG
      mkdir -p $WORKDIR
      if [ $? -ne 0 ]; then
        echo "$WORKDIR: fail to create directory"
        exit 254
      fi
      ;;
    d)
      DATADIR=$OPTARG
      if [[ ! -d $DATADIR ]]; then
        echo "$DATADIR: not a directory"
        exit 43
      fi
      ;;
    *)
      echo "usage: $(basename $0) [-t type] [-d data] [-w working] [-j jobs] <arguments...>"
      exit 1
      ;;
  esac
done
shift $(($OPTIND - 1))

case ${TYPE,,} in
  tm | pt | pathtm)
    splitTM $DATADIR $WORKDIR $@
    ;;
  pp | pd | pdh)
    splitPP $DATADIR $WORKDIR
    ;;
  vmu | hr | hrd)
    splitVMU $DATADIR $WORKDIR
    ;;
  *)
  echo "unknown data type provided: $TYPE! try $(basename $0) -h for more help"
  exit 5
  ;;
esac
