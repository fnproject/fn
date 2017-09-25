import asyncio
import sys
import uvloop

from hotfn.http import request
from hotfn.http import response


class MyProtocol(asyncio.Protocol):

    def connection_made(self, transport):
        print('pipe opened', file=sys.stderr, flush=True)
        super(MyProtocol, self).connection_made(transport=transport)

    def data_received(self, data):
        print('received: {!r}'.format(data), file=sys.stderr, flush=True)
        data = data.decode()
        req = request.RawRequest(data)
        (method, url, dict_params,
         headers, http_version, req_data) = req.parse_raw_request()

        rs = response.RawResponse(
            http_version, 200, "OK",
            response_data=req_data)
        print(rs.dump(), file=sys.stdout, flush=True)
        super(MyProtocol, self).data_received(data)

    def connection_lost(self, exc):
        print('pipe closed', file=sys.stderr, flush=True)
        super(MyProtocol, self).connection_lost(exc)


if __name__ == "__main__":
    with open("/dev/stdin", "rb", buffering=0) as stdin:
        asyncio.set_event_loop_policy(uvloop.EventLoopPolicy())
        loop = asyncio.get_event_loop()
        try:
            stdin_pipe_reader = loop.connect_read_pipe(MyProtocol, stdin)
            loop.run_until_complete(stdin_pipe_reader)
            loop.run_forever()
        finally:
            loop.close()
