#! /bin/sh
set -e

function listFilesExit() {
	echo The following files need to have go fmt ran:
	echo $NEED_TO_FORMAT
	exit 1
}

FOLDERS=$(go list -f {{.Dir}} ./... | grep -v vendor)
for i in $FOLDERS
do
cd $i
FILES=$(ls *.go)
	for j in $FILES
	do
	#echo $i/$j
	ALL_FILES="$ALL_FILES $i/$j"
	done
done
#echo $ALL_FILES
NEED_TO_FORMAT="$(gofmt -l $ALL_FILES)"
[[ -z $NEED_TO_FORMAT ]] || listFilesExit
