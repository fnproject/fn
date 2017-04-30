# Copyright 2016 Iron.io
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


FROM docker:1.13.1-dind

RUN apk update && apk upgrade && apk add --no-cache ca-certificates

COPY entrypoint.sh /usr/local/bin/
COPY dind.sh /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

# USAGE: Add a CMD to your own Dockerfile to use this (NOT an ENTRYPOINT, so that this is called)
# CMD ["./runner"]
