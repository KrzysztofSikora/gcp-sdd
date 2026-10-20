"""FR-02 analysis rules; examples are copied from specs/requirements.md."""

from app.analysis import AnalysisResult, WordCount, analyze, words


def top(result: AnalysisResult) -> list[tuple[str, int]]:
    return [(w.word, w.count) for w in result.top_words]


def test_fr01_example() -> None:
    assert analyze("The cat and the hat.") == AnalysisResult(
        word_count=5,
        character_count=20,
        top_words=[
            WordCount("the", 2),
            WordCount("and", 1),
            WordCount("cat", 1),
            WordCount("hat", 1),
        ],
    )


def test_ac1_characters_are_code_points() -> None:
    assert analyze("héllo wörld").character_count == 11


def test_ac2_word_boundaries() -> None:
    text = "Don't stop—don't! 2024 is_here"
    assert words(text) == ["don't", "stop", "don't", "2024", "is", "here"]
    assert analyze(text).word_count == 6


def test_ac2_typographic_apostrophe_joins_words() -> None:
    assert words("l’été don’t") == ["l’été", "don’t"]


def test_ac3_case_insensitive() -> None:
    assert top(analyze("Go go GO")) == [("go", 3)]


def test_ac4_top_five_by_count_then_alphabetically() -> None:
    assert top(analyze("b a c b a d e f b")) == [("b", 3), ("a", 2), ("c", 1), ("d", 1), ("e", 1)]


def test_ac5_fewer_than_five_distinct_words() -> None:
    assert top(analyze("one two one")) == [("one", 2), ("two", 1)]


def test_ac6_no_words() -> None:
    assert analyze("!!! ???") == AnalysisResult(word_count=0, character_count=7, top_words=[])
