(ns corpus.forms
  (:require [clojure.string :as str]))

(defn alpha [xs]
  ;; comment
  (let [ys (filter odd? xs)
        m {:a 1 :b #{2 3}}]
    #_(debug ys)
    (map #(inc %) ys)))

(defmacro beta [& body]
  `(do ~@body ~(first body) '~'x @state))

(def ^:private gamma
  #?(:cljs 1 :clj (fn [x] (* x 2.5e3)) :default nil))

(defn delta [s]
  (str/join "," [\a \newline "q\"x" #"re" 1/2 0xFF :k ::k/x]))

#?@(:clj [(defn spliced [] 1)])

(defn epsilon []
  #inst "2020" #:ns{:a 1} (.method obj) (Klass. 1))
