#! /bin/bash

ALL=stats.csv
DAILY=daily.csv

function doStatTMAll() {
  FILE=$WORKDIR/$ALL
  rm -f $FILE

  comma select 6,4,7 \
    | comma group 1 count 1 sum 2 sum 3 \
    | comma eval '4=$3/($2+$3)' \
    | comma format '4:float:percent' '5:size:iec' > $FILE

  if [ $? -ne 0 ]; then
    return 1
  fi
}

function doStatTMDaily() {
  FILE=$WORKDIR/$DAILY
  rm -f $FILE

  comma select 6,1,4,7 \
    | comma format 2:date:%Y/%j \
    | comma group 1:2 count 1 sum 3 sum 4 \
    | comma eval '5=$4/($3+$4)' \
    | comma format '5:float:percent' '6:size:iec' > $FILE

    if [ $? -ne 0 ]; then
      return 1
    fi
}

function statTM() {
  which tmcat &> /dev/null
  if [ $? -ne 0 ]; then
    echo "tmcat is not installed"
    exit 432
  fi

  tmcat list -a -c $DATADIR | tee >(doStatTMAll) >(doStatTMDaily) > /dev/null
  if [ $? -ne 0 ]; then
    return 1
  fi
}

function doStatPPAll() {
  FILE=$WORKDIR/$ALL
  rm -f $FILE

  comma select 3,5: \
    | comma group 1:2 count 1 sum 3 \
    | comma format 4:size:iec > $FILE
}

function doStatPPDaily() {
  FILE=$WORKDIR/$DAILY
  rm -f $FILE
  
  comma select 1,3,5: \
    | comma format 1:date:%Y/%j \
    | comma group 1:3 count 2 sum 4 \
    | comma format 4:size:iec > $FILE
}

function statPP() {
  which ppcat &> /dev/null
  if [ $? -ne 0 ]; then
    echo "ppcat is not installed"
    exit 432
  fi

  ppcat list -a -c $DATADIR | tee >(doStatPPAll) >(doStatPPDaily) > /dev/null
}

function statVMU() {
  echo "stats HR: not yet implemented"
  exit 255
}


for cmd in "comma"; do
  which $cmd > /dev/null
  if [ $? -ne 0 ]; then
    echo "$cmd is not installed"
    exit 432
  fi
done

TYPE=TM
DATADIR=$TMPDIR
WORKDIR=$TMPDIR

while getopts :t:d:j: OPT; do
  case $OPT in
    t)
      TYPE=$OPTARG
      ;;
    d)
      DATADIR=$(realpath $OPTARG)
      if [[ ! -d $DATADIR ]]; then
        echo "$DATADIR: not a directory"
        exit 43
      fi
      ;;
    *)
      echo "usage: $(basename $0) [-n filename] [-t type] [-d data] [-j jobs] <arguments...>"
      exit 1
      ;;
  esac
done
shift $(($OPTIND - 1))

if [ ${#} -eq 0 ]; then
  WORKDIR=$DATADIR
else
  WORKDIR=$1
  mkdir -p $WORKDIR
  if [ $? -ne 0 ]; then
    echo "$WORKDIR: fail to create directory"
    exit 123
  fi
fi

case ${TYPE,,} in
  tm | pt | pathtm)
    statTM
    ;;
  pp | pd | pdh)
    statPP
    ;;
  vmu | hr | hrd)
    statVMU
    ;;
  *)
  echo "unknown data type provided: $TYPE! try $(basename $0) -h for more help"
  exit 5
  ;;
esac
if [ $? -ne 0 ]; then
  echo "unexpected error"
  exit 2
fi
