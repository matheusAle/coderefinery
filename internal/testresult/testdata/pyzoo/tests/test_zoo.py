import pytest


def test_pass():
    assert True


def test_fail():
    assert 1 == 2, "one is not two"


@pytest.mark.skip(reason="not today")
def test_skip():
    pass


@pytest.fixture
def broken():
    raise RuntimeError("fixture exploded")


def test_error(broken):
    pass


@pytest.mark.parametrize("x", [1, 2, "a b"])
def test_param(x):
    assert x


class TestGroup:
    def test_method(self):
        assert True

    class TestNested:
        def test_inner(self):
            assert True
