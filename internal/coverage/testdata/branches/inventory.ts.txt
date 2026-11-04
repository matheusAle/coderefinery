export interface Item {
  sku: string;
  price: number;
  qty?: number;
  tags?: string[];
}

export class Inventory {
  private items: Item[] = [];
  #audit: string[] = [];

  add(item: Item): number {
    if (item.price < 0 || !item.sku) {
      throw new Error("invalid item");
    }
    this.items.push(item);
    this.#record(`add ${item.sku}`);
    return this.items.length;
  }

  find(sku: string): Item | undefined {
    return this.items.find((item) => item.sku === sku);
  }

  quantity(sku: string): number {
    return this.find(sku)?.qty ?? 0;
  }

  value(): number {
    let total = 0;
    for (const item of this.items) {
      total += item.price * (item.qty ?? 1);
    }
    return total;
  }

  private restock(sku: string, amount: number): boolean {
    const item = this.find(sku);
    if (item && amount > 0) {
      item.qty = (item.qty ?? 0) + amount;
      return true;
    }
    return false;
  }

  #record(entry: string): void {
    if (this.#audit.length >= 100) {
      this.#audit.shift();
    }
    this.#audit.push(entry);
  }
}

export function label(item: Item): string {
  return item.tags?.length ? `${item.sku} [${item.tags.join(",")}]` : item.sku;
}

export const discount = (price: number, member: boolean): number =>
  member ? price * 0.9 : price;

export function shippingZone(country: string): number {
  switch (country) {
    case "US":
      return 1;
    case "CA":
    case "MX":
      return 2;
    default:
      return 3;
  }
}
