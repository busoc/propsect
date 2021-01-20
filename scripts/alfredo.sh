#! /bin/bash

USER=""
PASSWD=""
HOST=""
APILIST="alfresco/api/-default-/public/cmis/versions/1.1/browser/root/Sites/busoc-ops/documentLibrary/10-DATA%20PRESERVATION"
APINODE="alfresco/api/-default-/public/cmis/versions/1.1/atom/content"
CURL="$(which curl)"
JQ="$(which jq)"
CUT="$(which cut)"
FILTER='.objects[].object.properties | select(."cmis:objectTypeId"="cmis:document") | [(."cmis:objectId".value | split(";")[0]), ."cmis:name".value, ."cmis:contentStreamMimeType".value] | @csv'

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

PAYLOAD=$1
DIRECTORY=$2

if [ -z $DIRECTORY ];
then
	DIRECTORY="${PWD}/${PAYLOAD}"
fi
mkdir -p $DIRECTORY 2> /dev/null
if [ $? -ne 0 ];
then
	echo "fail to create directory $DIRECTORY"
	exit 2
fi

COMMA=","
URL="http://${HOST}/${APILIST}/${PAYLOAD}/Documents"
$CURL -X GET -u "${USER}:${PASSWD}" "$URL" 2> /dev/null | $JQ -r "$FILTER" | while read LINE; do
	LINE=${LINE//\"/}
	ID=$(echo $LINE | $CUT -d "$COMMA" -f 1)
	FILE=$(echo $LINE | $CUT -d "$COMMA" -f 2)
	MIME=$(echo $LINE | $CUT -d "$COMMA" -f 3)

	URL="http://${HOST}/${APINODE}?id=${ID}"
	$CURL -X GET -H "Accept: ${MIME}" -u "${USER}:${PASSWD}" -o "${DIRECTORY}/${FILE}" "${URL}" 2> /dev/null
done
