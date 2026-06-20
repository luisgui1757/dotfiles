type GreetingProps = {
  name: string;
};

export function Greeting(props: GreetingProps) {
  return <span>Hello {props.name}</span>;
}
