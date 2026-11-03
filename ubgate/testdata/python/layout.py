# leading comment


def a(x):
    return x
    # trailing comment inside the block


class A:
    def m(self):
        pass
      # col 6: class block
    # col 4: class block
# col 0: module

if x:  # header comment
    # before first statement
    pass
# between if and else
else:
    pass

def one(): return 1  # one-line suite comment

@decorator  # decorator comment
def decorated():
    pass

x = [
    1,  # inside list
    2,
]
	# tab-indented comment
if y:
	z = 1
	if w:
		pass
