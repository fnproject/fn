# All Rights Reserved.
#
#    Licensed under the Apache License, Version 2.0 (the "License"); you may
#    not use this file except in compliance with the License. You may obtain
#    a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
#    Unless required by applicable law or agreed to in writing, software
#    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
#    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
#    License for the specific language governing permissions and limitations
#    under the License.

from hotfn.http import worker


@worker.coerce_input_to_content_type
def app(context, data=None, loop=None):
    """
    User's request body handler
    :param context: request context
    :type context: hotfn.http.request.RequestContext
    :param data: request body, it's type depends on request Content-Type header
    :type data: object
    :param loop: asyncio event loop
    :type loop asyncio.AbstractEventLoop
    :return: echo of data
    :rtype: object
    """
    return data


if __name__ == "__main__":
    worker.run(app)
