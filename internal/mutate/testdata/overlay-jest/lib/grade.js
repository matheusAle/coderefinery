function grade(score) {
  if (score >= 50) {
    return "pass";
  }
  return "fail";
}

module.exports = { grade };
