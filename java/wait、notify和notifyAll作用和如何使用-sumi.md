## wait、notify和notifyAll关键字    
这是java多线程中关键字，可以用来实现进程中的通信。  
以生产者消费者为例：  
  java程序中有两个线程--生产者和消费者，生产者产生数据，消费者消耗数据。在程序运行过程中，生产者需要通知消费者，让消费者去消耗数据，因为队列
  缓冲区（不为空）中有内容待消费。相应的，消费者需要通知生产者去生产更多的数据，因为当它消费掉一些数据后，缓冲区不再为满。  
  这里我们可以使用wait()来让一个线程在某些条件下暂停运行。例如，生产者线程在缓冲区为满，消费者在缓冲区为空时，暂停运行。  
  如果某些线程是等待某些条件而触发，当这些条件为真时，可以用notify和notifyAll来同通知那些正在等待中的线程重新开始运行。
  **不同之处在于，**notify仅仅是通知一个线程，并且我们并不会知道哪个线程会收到通知，而notifyAll
  是会通知所有等待中的线程。  
  简单来说，如果只有一个线程在等待一个信号，notify和notifyAll都会通知到这个线程，但是如果是多个线程
  在等待这个信号，notify只会通知到其中一个线程，其他线程不会受到通知，而notifyAll会唤醒所有等待中的线程。  
  - - -
  
  ## 如何使用wait  
 - wait()的作用是让当前线程进入等待状态，同时，wait()也会让当前线程释放它所持有的锁。
 直到其他线程调用此对象的 notify() 方法或 notifyAll() 方法，当前线程被唤醒，进入“就绪状态”。
- wait(long timeout)让当前线程处于“等待(阻塞)状态”，
直到其他线程调用此对象的notify()方法或 notifyAll() 方法，或者超过指定的时间量，当前线程被唤醒，进入“就绪状态”。  
- - -  
```
//main(主线程)
synchronized(t1){//这里t1是一个线程
       try {
              t1.start();
              t1.wait();
       } catch(InterruptedException e) {
    
              e.printStackTrace();
    
       }
}
```  
```
//唤醒主线程在t1线程中
   synchronized (this) {  //这里的 this 为 t1
          this.notify();
   }
```  
这里：  
1. synchronized(t1)锁定t1（获得t1的监视器）  
2. synchronized(t1)这里的锁定了t1，那么wait需用t1.wait()（释放掉t1）  
3. 因为wait需释放锁，所以必须在synchronized中使用（没有锁定则么可以释放？
没有锁时使用会抛出IllegalMonitorStateException（正在等待的对象没有锁））  
4. notify也要在synchronized使用，应该指定对象，t1. notify()，通知t1对象的等待池里的线程使一个线程进入锁定池，然后与锁定池中的线程争夺锁。那么为什么要在synchronized使用呢？
 t1. notify()需要通知一个等待池中的线程，那么这时我们必须得获得t1的监视器（需要使用synchronized），才能对其操作，
 t1. notify()程序只是知道要对t1操作，但是是否可以操作与是否可以获得t1锁关联的监视器有关。  
5. synchronized(),wait,notify() 对象一致性  
6. 在while循环里而不是if语句下使用wait（防止虚假唤醒spurious wakeup）  
- - -  
证明wait可以使当前线程等待
```
class ThreadA extends Thread{
    public ThreadA(String name) {
        super(name);
    }
    public void run() {
        synchronized (this) {
            try {                       
                Thread.sleep(1000); //  使当前线阻塞 1 s，确保主程序的 t1.wait(); 执行之后再执行 notify()
            } catch (Exception e) {
                e.printStackTrace();
            }           
            System.out.println(Thread.currentThread().getName()+" call notify()");
            // 唤醒当前的wait线程
            this.notify();
        }
    }
}
public class WaitTest {
    public static void main(String[] args) {
        ThreadA t1 = new ThreadA("t1");
        synchronized(t1) {
            try {
                // 启动“线程t1”
                System.out.println(Thread.currentThread().getName()+" start t1");
                t1.start();
                // 主线程等待t1通过notify()唤醒。
                System.out.println(Thread.currentThread().getName()+" wait()");
                t1.wait();  //  不是使t1线程等待，而是当前执行wait的线程等待
                System.out.println(Thread.currentThread().getName()+" continue");
            } catch (InterruptedException e) {
                e.printStackTrace();
            }
        }
    }
}
```  
在代码中，使用wait注意两个问题：  
1. 在代码中，我们是对在多线程间共享的那个Object来使用wait。在生产者消费者问题中，
这个共享的Object就是那个缓冲区队列。  
2. 我们应该在synchronized的函数或是对象里调用wait，但哪个对象应该被synchronized呢？
答案是，那个你希望上锁的对象就应该被synchronized，即那个在多个线程间被共享的对象。
在生产者消费者问题中，应该被synchronized的就是那个缓冲区队列。  
  
 ## notify与notifyAll
 notify()和notifyAll()的作用，则是唤醒当前对象上的等待线程；
 notify()是唤醒单个线程，而notifyAll()是唤醒所有的线程。  
 在上述wait的代码示例中也看到了notify的具体使用。  
 - - - 
 
 ## 永远在循环（loop）里调用 wait 和 notify，不是在 If 语句
 我们知道wait是应该永远在被synchronized的背景下和那个被多线程共享的对象上调用。
 同时，针对于，线程是在某些条件下等待，我们应该使用while循环，而不是在if语句中调用wait。
 以上述生产者消费者为例，即“如果缓冲区队列是满的话，那么生产者线程应该等待”，你可能直觉就
 会写一个if语句。但if语句存在一些微妙的小问题，导致即使条件没被满足，你的线程你也有可能
 被错误地唤醒。所以如果你不在线程被唤醒后再次使用while循环检查唤醒条件是否被满足，你的程
 序就有可能会出错——例如在缓冲区为满的时候生产者继续生成数据，或者缓冲区为空的时候消费者开
 始消耗数据。所以记住，永远在while循环而不是if语句中使用wait！  
 **在while循环里使用wait的目的，是在线程被唤醒的前后都持续检查条件是否被满足。**  
 
 ## Java wait(), notify(), notifyAll() 范例
我们有两个线程，分别名为PRODUCER（生产者）和CONSUMER（消费者），他们分别继承了了Producer
和Consumer类，而Producer和Consumer都继承了Thread类。Producer和Consumer想要实现的代码逻
辑都在run()函数内。Main线程开始了生产者和消费者线程，并声明了一个LinkedList作为缓冲区队列
（在Java中，LinkedList实现了队列的接口）。生产者在无限循环中持续往LinkedList里插入随机整数
直到LinkedList满。我们在while(queue.size == maxSize)循环语句中检查这个条件。请注意到我们
在做这个检查条件之前已经在队列对象上使用了synchronized关键词，因而其它线程不能在我们检查条件
时改变这个队列。如果队列满了，那么PRODUCER线程会在CONSUMER线程消耗掉队列里的任意一个整数，
并用notify来通知PRODUCER线程之前持续等待。  
在这里，wait和notify都是使用在同一个共享对象上的。  
```
importjava.util.LinkedList; 
importjava.util.Queue; 
importjava.util.Random; 

publicclass ProducerConsumerInJava { 
    publicstatic void main(String args[]) { 
        System.out.println("How to use wait and notify method in Java");
        System.out.println("Solving Producer Consumper Problem");
        Queue<Integer> buffer = newLinkedList<>(); 
        intmaxSize = 10;
        Thread producer = newProducer(buffer, maxSize, "PRODUCER");
        Thread consumer = newConsumer(buffer, maxSize, "CONSUMER");
        producer.start(); consumer.start(); } 
    }
    classProducer extendsThread 
    {privateQueue<Integer> queue; 
        privateint maxSize; 
        publicProducer(Queue<Integer> queue, intmaxSize, String name){ 
            super(name);this.queue = queue; this.maxSize = maxSize; 
        }
        @Overridepublic void run() 
        {
            while(true)
                {
                    synchronized(queue) { 
                        while(queue.size() == maxSize) { 
                            try{ 
                                System.out .println("Queue is full, " + "Producer thread waiting for "+ "consumer to take something from queue");
                                queue.wait();
                            }catch(Exception ex) { 
                                ex.printStackTrace(); } 
                            }
                            Random random = newRandom(); 
                            inti = random.nextInt(); 
                            System.out.println("Producing value : " + i); queue.add(i); queue.notifyAll(); 
                        }
                    }
                }
            }
    classConsumer extendsThread { 
        privateQueue<Integer> queue; 
        privateint maxSize; 
        publicConsumer(Queue<Integer> queue, intmaxSize, String name){ 
            super(name);
            this.queue = queue; 
            this.maxSize = maxSize; 
        }
        @Overridepublic void run() { 
            while(true) { 
                synchronized(queue) { 
                    while(queue.isEmpty()) { 
                        System.out.println("Queue is empty," + "Consumer thread is waiting" + " for producer thread to put something in queue");
                        try{ 
                            queue.wait();
                        }catch(Exception ex) { 
                            ex.printStackTrace();
                        }
                    }
                    System.out.println("Consuming value : " + queue.remove()); queue.notifyAll(); 
                }
            }
        }
    }
``` 