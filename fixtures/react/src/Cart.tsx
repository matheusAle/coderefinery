import { useEffect, useState } from "react";

export interface CartItem {
  id: string;
  price: number;
  quantity: number;
}

interface CartProps {
  items: CartItem[];
  currency: string;
  couponCode?: string;
  onCheckout: (total: number) => void;
}

export function Cart({ items, currency, couponCode, onCheckout }: CartProps) {
  const [total, setTotal] = useState(0);
  const [discount, setDiscount] = useState(0);
  const [warning, setWarning] = useState("");
  const [label, setLabel] = useState(currency);

  useEffect(() => {
    let sum = 0;
    for (const item of items) {
      if (item.quantity <= 0) continue;
      sum += item.price * item.quantity;
    }
    let off = 0;
    if (couponCode === "HALF") off = sum / 2;
    else if (couponCode === "TEN" && sum >= 10) off = 10;
    if (sum > 1000 || items.length > 50) {
      setWarning("large order");
    } else if (sum === 0) {
      setWarning(items.length ? "only empty lines" : "empty cart");
    } else {
      setWarning("");
    }
    setDiscount(off);
    setTotal(sum - off);
  }, [items, couponCode]);

  useEffect(() => {
    setLabel(currency.toUpperCase());
  }, [currency]);

  const handleCheckout = () => {
    if (total > 0) onCheckout(total);
  };

  function handleClear() {
    setTotal(0);
  }

  if (items.length === 0) return <p>Your cart is empty</p>;

  return (
    <div>
      <ul>
        {items.map((item) => (
          <li key={item.id}>
            {item.id} x{item.quantity}
          </li>
        ))}
      </ul>
      {discount > 0 && <p>Discount: {discount}</p>}
      {warning ? <p role="alert">{warning}</p> : null}
      <p>
        Total: {total} {label}
      </p>
      <button onClick={handleCheckout}>Checkout</button>
      <button onClick={handleClear}>Clear</button>
    </div>
  );
}
