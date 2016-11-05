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

require 'json'

payload = STDIN.read
if payload != ""
  payload = JSON.parse(payload)
  
  # payload contains checks
  if payload["sleep"] 
    i = payload['sleep'].to_i
    puts "Sleeping for #{i} seconds..."
    sleep i
    puts "I'm awake!"
  end
end
