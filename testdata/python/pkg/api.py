from .store import fetch_user


class Worker:
    """Process incoming names."""
    label: str

    def run(self, name: str) -> str:
        return fetch_user(name)


async def handler(name: str) -> str:
    return fetch_user(name)
