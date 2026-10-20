"""HTTP layer of the analyzer (specs/design.md, private service contract)."""

from typing import Annotated

from fastapi import FastAPI
from pydantic import BaseModel, ConfigDict, Field, StrictStr, field_validator

from app.analysis import MAX_TEXT_LENGTH, analyze


class AnalyzeRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    text: Annotated[StrictStr, Field(min_length=1, max_length=MAX_TEXT_LENGTH)]

    @field_validator("text")
    @classmethod
    def not_blank(cls, value: str) -> str:
        if not value.strip():
            raise ValueError("text must not be empty or whitespace only")
        return value


class WordCountOut(BaseModel):
    word: str
    count: int


class AnalyzeResponse(BaseModel):
    word_count: int
    character_count: int
    top_words: list[WordCountOut]


class Health(BaseModel):
    status: str


app = FastAPI(title="text-analyzer", docs_url=None, redoc_url=None, openapi_url=None)


@app.post("/analyze")
def analyze_text(request: AnalyzeRequest) -> AnalyzeResponse:
    result = analyze(request.text)
    return AnalyzeResponse(
        word_count=result.word_count,
        character_count=result.character_count,
        top_words=[WordCountOut(word=w.word, count=w.count) for w in result.top_words],
    )


@app.get("/healthz")
def healthz() -> Health:
    return Health(status="ok")
