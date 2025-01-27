import anthropic


def new_client(provider="anthropic"):
    if provider != "anthropic":
        raise ValueError("Unknown provider: %s" % provider)
    client = anthropic.Anthropic()
    return client
