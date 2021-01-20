#! /bin/bash

COMMA=","
APILIST="alfresco/api/-default-/public/cmis/versions/1.1/browser/root/Sites/busoc-ops/documentLibrary/10-DATA%20PRESERVATION"
APINODE="alfresco/api/-default-/public/cmis/versions/1.1/atom/content"
CURL="$(which curl)"
JQ="$(which jq)"
CUT="$(which cut)"

if [ ! -x $CURL ];
then
	echo "missing curl command"
	exit 1
fi

if [ ! -x $JQ ];
then
	echo "missing jq command"
	exit 1
fi

if [ ! -x $CUT ];
then
	echo "missing cut command"
	exit 1
fi

FILTER='.objects[].object.properties | [(."cmis:objectId".value | split(";")[0]), ."cmis:name".value, ."cmis:contentStreamMimeType".value, (."cmis:objectTypeId".value | split(":")[1])] | @csv'
# FILTER='.objects[].object.properties | select(."cmis:objectTypeId"="cmis:document") | [(."cmis:objectId".value | split(";")[0]), ."cmis:name".value, ."cmis:contentStreamMimeType".value] | @csv'

DIRECTORY="$PWD"
WHAT=""
USER=""
PASSWD=""
HOST="localhost:8080"
PAYLOAD="00-Test"
RECURSE=0
while getopts :w:u:p:r:h:d: OPT; do
	case $OPT in
	w)
	WHAT=${OPTARG^}
	;;
	u)
	USER=$OPTARG
	;;
	p)
	PASSWD=$OPTARG
	;;
	h)
	HOST=$OPTARG
	;;
	r)
	RECURSE=1
	;;
	d)
	DIRECTORY=$OPTARG
	;;
	*)
	echo "$OPT not defined"
	echo "usage: $(basename $0) [-w what] [-u user] [-p passwd] [-r host] [-d directory] <remote directory>"
	exit 3
	;;
	esac
done
shift $(($OPTIND - 1))
PAYLOAD=$1


mkdir -p $DIRECTORY 2> /dev/null
if [ $? -ne 0 ];
then
	echo "fail to create directory $DIRECTORY"
	exit 2
fi

download() {
	local BASE=$1
	$CURL -X GET -u "${USER}:${PASSWD}" "$BASE" 2> /dev/null | $JQ -r "$FILTER" | while read LINE; do
		LINE=${LINE//\"/}
		ID=$(echo $LINE | $CUT -d "$COMMA" -f 1)
		FILE=$(echo $LINE | $CUT -d "$COMMA" -f 2)
		MIME=$(echo $LINE | $CUT -d "$COMMA" -f 3)
		TYPE=$(echo $LINE | $CUT -d "$COMMA" -f 4)

		if [ $RECURSE -eq 1 ] && [ $TYPE == "folder" ]; then
			download "$BASE/$FILE"
			continue
		fi
		URL="http://${HOST}/${APINODE}?id=${ID}"
		echo "downloading $FILE"
		$CURL -X GET -H "Accept: ${MIME}" -u "${USER}:${PASSWD}" -o "${DIRECTORY}/${FILE}" "${URL}" 2> /dev/null
	done
}

download "http://${HOST}/${APILIST}/${PAYLOAD}/${WHAT}"
