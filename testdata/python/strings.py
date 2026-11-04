s1 = 'esc \n \t \\ \' \x41 \u1234 \U0001F600 \N{DASH} \777 \q'
s2 = b'\u1234 \N{X} \x00'
s3 = r'\\' + r'a\'b' + r"\""
s4 = rf'\{a}\.b' + rf'\\{x}' + rf'\{{lit}}'
s5 = f'{x!r:>{w}} {y=} {z:{{}}} {{esc}}'
s6 = '''multi
line \'still'''
s7 = ("implicit"
      'concat' f"{1 + 2}")
s8 = x + \
    'continued string'
s9 = u'uni' Rb'raw bytes' BR"x" F'{a}'
