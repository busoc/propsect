#! /bin/bash

BANDWIDTH=8000
FILE=sample.lst
SERVER=nobody@localhost
REMDIR=/
LOCAL=/storage
LOGDIR=logs

while getopts "b:f:d:s:l:r:" opt
do
        case ${opt} in
                b)
                        BANDWIDTH=$OPTARG
                        ;;
                f)
                        FILE=$OPTARG
                        ;;
                d)
                        LOCAL=$OPTARG
                        ;;
                s)
                        REMDIR=$OPTARG
                        ;;
                l)
                        LOGDIR=$OPTARG
                        ;;
                r)
                        SERVER=$OPTARG
                        ;;
                *)
                        echo "usage: syncdata.sh [-b bandwidth] [-f file] [-d dstdir] [-s srcdir] [-l logdir] [-r server]"
                        echo ""

                        echo "example: syncdata.sh -b $BANDWIDTH -l $LOGDIR -f $FILE -s $REMDIR -d $LOCAL -r $SERVER"
                        exit 2
                        ;;
        esac
done

mkdir -p $LOCAL;
if [ $? -ne 0 ]; then
        echo "fail to create directory $LOCAL";
        exit 1;
fi

mkdir -p $LOGDIR;
if [ $? -ne 0 ]; then
        echo "fail to create directory $LOGDIR";
        exit 1;
fi

REMOTE=$SERVER:$REMDIR
LOGFILE=$LOGDIR/$(basename $FILE)-$(date +%Y%j_%H%M%S).log

/usr/bin/rsync --bwlimit $BANDWIDTH -a -vv --log-file=$LOGFILE --files-from $FILE $REMOTE $LOCAL
