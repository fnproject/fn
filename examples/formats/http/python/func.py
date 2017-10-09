import fdk


@fdk.coerce_http_input_to_content_type
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
    fdk.handle(app)
