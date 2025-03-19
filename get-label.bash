#!/bin/bash
#First set the suffix to some value, either Teamcity build-id or <abbrev-sha1>
if [[ -z "${TEAMCITY_BUILD_ID}" ]]; then
    SUFFIX=$(git rev-parse --short HEAD)
else
    SUFFIX=${TEAMCITY_BUILD_ID}
fi

# Get the tag as the version
TAG=$(git describe --tags HEAD)
if [[ $? -ne 0 ]]
then
    # if we cannot get the tag, lets use the <branch>-pmk-<suffix> as the tag name
    TAG=$(git rev-parse --abbrev-ref HEAD | sed 's/[^a-zA-Z0-9_.]/-/g')-kaapi-${SUFFIX}
else
    TAG=$(echo $TAG | sed 's/-.*//')-kaapi-${SUFFIX}
fi
echo $TAG