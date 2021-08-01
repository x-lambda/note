import numpy as np

"""
马尔科夫过程
马尔科夫奖励过程
价值函数
策略

马尔科夫链(过程)是通常用一组随机变量定义的数学系统，可以根据具体的概率规则进行状态转移。转移的集合满足马尔科夫性质。
也就是说，转移到任一特定状态的概率只取决于当前状态和所用时间，而与其之前的状态序列无关。
马尔科夫链的这个独特性质就是无记忆性。

从计算机的角度看：马尔科夫过程就是个状态机，只不过伴随着概率分布
"""

# 状态空间 [有限]
states = ["Sleep", "IceCream", "Run"]

# 可能的事件序列
transition_name = [["SS", "SR", "SI"], ["RS", "RR", "RI"], ["IS", "IR", "II"]]

# 概率矩阵 (转移矩阵)
transition_matrix = [[0.2, 0.6, 0.2], [0.1, 0.6, 0.3], [0.2, 0.7, 0.1]]


def activity_forecast(days):
    """实现了可以预测状态的马尔科夫模型的函数
    :param days:
    :return:
    """
    # 初始状态
    activity_today = "Sleep"
    print("Start state: " + activity_today)

    # 应该记录选择的状态序列，这里现在只有初始状态
    activity_list = [activity_today]

    i = 0
    prob = 1  # 计算 activity_list 的概率
    while i != days:
        if activity_today == "Sleep":
            # 从可能的转移集合中选出随机样本
            # p: 样本集的概率分布
            # 从可能发生的样本集中，根据其概率分布，随机选择一个事件
            # 样本集:       ["SS", "SR", "SI"]
            # 对应的概率分布: [0.2,  0.6,  0.2]
            # 即有20%的可能性返回SS 有60%的可能性返回SR 有20%的可能性返回SI
            change = np.random.choice(transition_name[0], replace=True, p=transition_matrix[0])
            if change == "SS":
                prob = prob * 0.2
                activity_list.append("Sleep")
                # 状态转移至下一个
                activity_today = "Sleep"
            elif change == "SR":
                prob = prob * 0.6
                activity_list.append("Run")
                activity_today = "Run"
            else:
                prob = prob * 0.2
                activity_list.append("IceCream")
                activity_today = "IceCream"
        elif activity_today == "Run":
            # 样本集:        ["RS", "RR", "RI"]
            # 对应的概率分布:  [0.1,  0.6,  0.3]
            # 即有10%的概率返回RS  有60%的概率返回RR  有30%的概率返回RI
            change = np.random.choice(transition_name[1], replace=True, p=transition_matrix[0])
            if change == "RS":
                prob = prob * 0.1
                activity_list.append("Sleep")
                activity_today = "Sleep"
            elif change == "RR":
                prob = prob * 0.6
                activity_list.append("Run")
                activity_today = "Run"
            else:
                prob = prob * 0.3
                activity_list.append("IceCream")
                activity_today = "IceCream"
        elif activity_today == "IceCream":
            # 状态样本集:           ["IS", "IR", "II"]
            # 对应的状态转移概率分布:  [0.2,  0.7,  0.1]
            # 即有20%的概率返回IS  有70%的概率返回IR  有10%的概率返回II
            change = np.random.choice(transition_name[2], replace=True, p=transition_matrix[0])
            if change == "IS":
                prob = prob * 0.2
                activity_list.append("Sleep")
                activity_today = "Sleep"
            elif change == "IR":
                prob = prob * 0.7
                activity_list.append("Run")
                activity_today = "Run"
            else:
                prob = prob * 0.1
                activity_list.append("IceCream")
                activity_today = "IceCream"

        # 往前推进一个时间
        i += 1

    print("Possible states: " + str(activity_list))
    print("End state after " + str(days) + " days: " + activity_today)
    print("Probability of possible sequence of states: " + str(prob))
    print("*" * 80)


def main():
    for i in range(len(transition_matrix)):
        if abs(sum(transition_matrix[i]) - 1) > 0.000001:
            raise Exception("Transition matrix error")

    # 查看2天后的状态，及其中间状态转移
    activity_forecast(2)

    # 查看5天后的状态，及其中间状态转移
    activity_forecast(5)


if __name__ == "__main__":
    main()
