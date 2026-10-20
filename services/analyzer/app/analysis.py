"""Text analysis rules (specs/requirements.md FR-02). Pure functions, no HTTP concerns."""

import re
from collections import Counter
from dataclasses import dataclass

MAX_TEXT_LENGTH = 10_000
TOP_WORDS_LIMIT = 5

# A run of Unicode letters or digits; inner apostrophes (' or ’) join parts, as in "don't".
WORD = re.compile(r"[^\W_]+(?:['’][^\W_]+)*")


@dataclass(frozen=True)
class WordCount:
    word: str
    count: int


@dataclass(frozen=True)
class AnalysisResult:
    word_count: int
    character_count: int
    top_words: list[WordCount]


def words(text: str) -> list[str]:
    """Lower-cased words of the text, in order of appearance."""
    return WORD.findall(text.lower())


def analyze(text: str) -> AnalysisResult:
    found = words(text)
    ranked = sorted(Counter(found).items(), key=lambda item: (-item[1], item[0]))
    return AnalysisResult(
        word_count=len(found),
        character_count=len(text),
        top_words=[WordCount(word, count) for word, count in ranked[:TOP_WORDS_LIMIT]],
    )
