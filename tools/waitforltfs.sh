#!/bin/sh

begin=`date +%s`
while pgrep -x ltfs > /dev/null; do
	sleep 0.5
done
end=`date +%s`

echo "done, waited $(( $end - $begin )) seconds"

