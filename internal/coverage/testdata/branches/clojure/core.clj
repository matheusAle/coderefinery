(ns demo.core)

(defn sign [x]
  (if (pos? x)
    :positive
    (when (neg? x) :negative)))

(defn label [x]
  (cond
    (= x 1) "one"
    (= x 2) "two"
    :else "many"))

(defn unused [x]
  (if x 1 0))
