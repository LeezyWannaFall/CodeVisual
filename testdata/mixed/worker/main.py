class Queue:
    """A Python worker beside the Go application."""
    name: str

    def consume(self, message: str) -> str:
        return message.strip()


def process(message: str) -> str:
    return message.upper()
