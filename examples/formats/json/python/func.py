import fdk


def handler(context, data=None, loop=None):
    """
    This is just an echo function
    :param context: request context
    :type context: hotfn.http.request.RequestContext
    :param data: request body
    :type data: object
    :param loop: asyncio event loop
    :type loop: asyncio.AbstractEventLoop
    :return: echo of request body
    :rtype: object
    """
    return data


if __name__ == "__main__":
    fdk.handle(handler)
