import asyncio
import json
import sys
import uvloop


class JSONProtocol(asyncio.Protocol):

    def connection_made(self, transport):
        print('pipe opened', file=sys.stderr, flush=True)
        super(JSONProtocol, self).connection_made(transport)

    def data_received(self, data):
        try:
            print('received: {!r}'.format(data),
                  file=sys.stderr, flush=True)
            dict_data = json.loads(data.decode())
            body_obj = dict_data['body']
            print("body type: {}".format(type(body_obj)), file=sys.stderr, flush=True)
            if isinstance(body_obj, str):
                body = json.loads(body_obj)
            else:
                body = body_obj
            print("body loaded: {}".format(body), file=sys.stderr, flush=True)
            inner = json.dumps({
                "data": body['data'],
            })
            out_data = {
                "body": inner,
                "status_code": 202
            }
            new_data = json.dumps(out_data)
            print(new_data, file=sys.stderr, flush=True)
            print(new_data, file=sys.stdout, flush=True)
            super(JSONProtocol, self).data_received(data)
        except (Exception, BaseException) as ex:
            err = json.dumps({
                "error": {
                    "message": str(ex)
                }
            })
            print(err, file=sys.stdout, flush=True)

    def connection_lost(self, exc):
        print('pipe closed', file=sys.stderr, flush=True)
        super(JSONProtocol, self).connection_lost(exc)


if __name__ == "__main__":
    with open("/dev/stdin", "rb", buffering=0) as stdin:
        asyncio.set_event_loop_policy(uvloop.EventLoopPolicy())
        loop = asyncio.get_event_loop()
        try:
            stdin_pipe_reader = loop.connect_read_pipe(JSONProtocol, stdin)
            loop.run_until_complete(stdin_pipe_reader)
            loop.run_forever()
        finally:
            loop.close()
