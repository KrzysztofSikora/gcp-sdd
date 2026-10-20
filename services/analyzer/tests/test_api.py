"""FR-05 and FR-06 through the HTTP layer."""

import pytest
from fastapi.testclient import TestClient

from app.main import app

client = TestClient(app)


def test_fr05_ac1_analyze() -> None:
    response = client.post("/analyze", json={"text": "a b a"})
    assert response.status_code == 200
    assert response.json() == {
        "word_count": 3,
        "character_count": 5,
        "top_words": [{"word": "a", "count": 2}, {"word": "b", "count": 1}],
    }


def test_contract_example() -> None:
    response = client.post("/analyze", json={"text": "The cat and the hat."})
    assert response.json() == {
        "word_count": 5,
        "character_count": 20,
        "top_words": [
            {"word": "the", "count": 2},
            {"word": "and", "count": 1},
            {"word": "cat", "count": 1},
            {"word": "hat", "count": 1},
        ],
    }


def test_max_length_is_accepted() -> None:
    assert client.post("/analyze", json={"text": "a" * 10_000}).status_code == 200


@pytest.mark.parametrize(
    "body",
    [
        {},
        {"text": None},
        {"text": 42},
        {"text": ""},
        {"text": " \n\t "},
        {"text": "a" * 10_001},
        {"text": "ok", "extra": 1},
    ],
    ids=["missing", "null", "number", "empty", "blank", "too-long", "extra-field"],
)
def test_fr05_ac2_invalid_input_is_422(body: dict) -> None:
    assert client.post("/analyze", json=body).status_code == 422


def test_fr06_healthz() -> None:
    response = client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}
